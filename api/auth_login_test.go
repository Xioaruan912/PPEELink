package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"ppeelink/middlewares"
	"ppeelink/models"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestUserLoginWithCredentialsOnly(t *testing.T) {
	oldDB := models.DB
	oldSecret := middlewares.Secret
	oldGinMode := gin.Mode()
	t.Cleanup(func() {
		models.DB = oldDB
		middlewares.Secret = oldSecret
		gin.SetMode(oldGinMode)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	models.DB = db
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	if err := (&models.User{Username: "admin", Password: "secret-123"}).Create(); err != nil {
		t.Fatal(err)
	}
	middlewares.Secret = []byte("test-login-secret")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/auth/login", UserLogin)
	form := url.Values{"username": {"admin"}, "password": {"secret-123"}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("credential-only login returned %d: %s", recorder.Code, recorder.Body.String())
	}
}
