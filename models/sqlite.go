package models

import (
	"log"
	"os"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB
var isInitialized bool

func InitSqlite() {
	if isInitialized {
		log.Println("数据库已经初始化，无需重复初始化")
		return
	}
	// 确保数据库目录存在
	if err := os.MkdirAll("./db", 0o755); err != nil {
		log.Println("创建数据库目录失败:", err)
	}
	// WAL + busy_timeout + 32MB page cache + 128MB mmap + memory temp_store
	dsn := "./db/ppeelink.db?_pragma=busy_timeout(8000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)&_pragma=cache_size(-32000)&_pragma=temp_store(MEMORY)&_pragma=mmap_size(134217728)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	DB = db
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		sqlDB.SetMaxOpenConns(8)
		sqlDB.SetMaxIdleConns(4)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}
	err = db.AutoMigrate(&User{}, &Subcription{}, &SubLogs{}, &GroupNode{}, &Node{},
		&Probe{}, &ProbeResult{}, &ClashUnlockSnapshot{}, &NetworkTraceRun{}, &NetworkTraceView{}, &ThroughputRun{}, &ThroughputJob{}, &ThroughputCapability{}, &TraceProbeCapability{}, &TraceAgentCapability{}, &Monitor{}, &MonitorMetric{}, &NodeQualitySample{}, &NodeTargetQualitySample{}, &NodeHealthEvent{}, &AlertSetting{}, &WebDAVBackupSetting{}, &UnlockObservation{}, &TemplateVersion{}, &RuleSource{}, &RuleCatalog{}, &RuleCacheSnapshot{}, &EgressTarget{}, &TaskRun{}, &SubscriptionArtifact{}, &SubscriptionArtifactPointer{}, &APIToken{}, &RoutingRegressionCase{}, &AuditLog{}, &ShortLink{}, &ProxyServer{}, &AgentRelay{}, &AgentRelayDay{}, &ProxyInbound{}, &ProxyTraffic{}, &ProxyAccount{}, &WebsiteTraffic{}, &WebsiteTrafficReceipt{})
	if err != nil {
		log.Println("数据表迁移失败:", err)
	}
	// 高频时序与统计查询复合索引（幂等创建，加速监控、网站流量与节点质量查询）
	for _, ddl := range []string{
		"CREATE INDEX IF NOT EXISTS idx_monitor_metrics_mid_time ON monitor_metrics(monitor_id, collected_at)",
		"CREATE INDEX IF NOT EXISTS idx_node_quality_nid_time ON node_quality_samples(node_id, checked_at)",
		"CREATE INDEX IF NOT EXISTS idx_website_traffic_srv_bucket ON website_traffics(server_id, bucket_start)",
		"CREATE INDEX IF NOT EXISTS idx_probe_results_node_time ON probe_results(node_id, checked_at)",
	} {
		_ = db.Exec(ddl).Error
	}
	if err := RemoveClientProxyChains(); err != nil {
		log.Fatalf("清理已移除的客户端链式代理设置失败: %v", err)
	}
	if err := MigrateNodeVisibilityToDelivery(); err != nil {
		log.Fatalf("迁移节点下发设置失败: %v", err)
	}
	_ = RecoverInterruptedTasks()
	if err := EnsureDefaultEgressTargets(); err != nil {
		log.Println("初始化分流检测目标失败:", err)
	}
	EnsureAllTokens()
	if err := EnsureNodeGroupMembership(); err != nil {
		log.Println("修复未分组节点失败:", err)
	}
	// 初始化用户数据
	err = db.First(&User{}).Error
	if err == gorm.ErrRecordNotFound {
		admin := &User{
			Username:           "admin",
			Password:           DefaultPassword,
			Role:               RoleAdmin,
			Nickname:           "管理员",
			MustChangePassword: true,
		}
		err = admin.Create()
		if err != nil {
			log.Println("初始化添加用户数据失败")
		}
	}
	// 设置初始化标志为 true
	isInitialized = true
	log.Println("数据库初始化成功") // 只有在没有任何错误时才会打印这个日志
}

// OptimizeSqlite runs SQLite's query planner statistics refresh.
func OptimizeSqlite() {
	if DB == nil {
		return
	}
	_ = DB.Exec("PRAGMA optimize").Error
}

// CloseSqlite checkpoints the WAL file cleanly and closes the database pool on shutdown.
func CloseSqlite() {
	if DB == nil {
		return
	}
	_ = DB.Exec("PRAGMA optimize").Error
	_ = DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error
	if sqlDB, err := DB.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

// ReopenSqlite closes and reopens the SQLite database after an in-place full backup restore.
func ReopenSqlite() error {
	CloseSqlite()
	dsn := "./db/ppeelink.db?_pragma=busy_timeout(8000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)&_pragma=cache_size(-32000)&_pragma=temp_store(MEMORY)&_pragma=mmap_size(134217728)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		sqlDB.SetMaxOpenConns(8)
		sqlDB.SetMaxIdleConns(4)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}
	DB = db
	return nil
}
