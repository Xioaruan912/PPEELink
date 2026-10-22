export default {
  // 路由国际化
  route: {
    dashboard: "Dashboard",
    document: "Document",
    userset: "ChangePassword",
    system: "system management",
    nodelist: "Node List",
    sublist: "Subscription List",
    subcription: "Subscription Management",
    templatelist: "Template List",
    template: "Template",
    templatebuilder: "Template Builder",
    rulecenter: "Rule Center",
    unlocktest: "Unlock Test",
    test: "Test",
    tcptest: "TCP Test",
  },
  // 登录页面国际化
  login: {
    username: "Username",
    password: "Password",
    login: "Login",
    capsLock: "Caps Lock is On",
    message: {
      username: {
        required: "Please enter Username",
      },
      password: {
        required: "Please enter Password",
        min: "The password can not be less than 6 digits",
      },
    },
  },
  // 重置密码页面国际化
  userset: {
    title: "Change Password",
    newUsername: "New Username",
    newPassword: "New Password",
    message: {
      title: "Prompt",
      xx1: "Username or password cannot be empty",
      xx2: "The password length cannot be less than 6 digits",
      xx3: "Are you sure you want to reset the password",
      xx4: "Password reset successful, new password is:",
    },
  },
};
