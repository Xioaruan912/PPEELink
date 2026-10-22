export default {
  // 路由国际化
  route: {
    dashboard: "首页",
    document: "项目文档",
    userset: "修改密码",
    system: "系统管理",
    nodelist: "节点列表",
    sublist: "订阅列表",
    subcription: "订阅管理",
    templatelist: "模板列表",
    template: "模板",
    templatebuilder: "操作模板",
    rulecenter: "规则中心",
    unlocktest: "解锁测试",
    test: "测试",
    tcptest: "TCP测试",
  },
  // 登录页面国际化
  login: {
    username: "用户名",
    password: "密码",
    login: "登 录",
    capsLock: "大写锁定已打开",
    message: {
      username: {
        required: "请输入用户名",
      },
      password: {
        required: "请输入密码",
        min: "密码不能少于6位",
      },
    },
  },
  // 重置密码页面国际化
  userset: {
    title: "修改密码",
    newUsername: "新账号",
    newPassword: "新密码",
    message: {
      title: "提示",
      xx1: "账号或密码不能为空",
      xx2: "密码长度不能小于6位",
      xx3: "你确定要重置密码吗",
      xx4: "密码重置成功，新密码是：",
    },
  },
};
