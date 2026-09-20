import { defineMock } from "./base";

export default defineMock([
  {
    url: "auth/login",
    method: ["POST"],
    body: {
      code: "00000",
      data: {
        accessToken:
          "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhZG1pbiIsImp0aSI6IjE2YWJkNTlkOTAxNzQwZDliYmI3ZjczODBhZDkyNzNhIiwidXNlcklkIjoyLCJ1c2VybmFtZSI6ImFkbWluIiwiZGVwdElkIjoxLCJkYXRhU2NvcGUiOjEsImF1dGhvcml0aWVzIjpbIlJPTEVfQURNSU4iXSwiZXhwIjoxNjkxMTAzMzgyfQ.P4cuIfmPepl3HuguhMS7NXn5a7IUPpsLbmtA_rHOhHk",
        tokenType: "Bearer",
        refreshToken: null,
        expires: null,
      },
      msg: "一切ok",
    },
  },

  {
    url: "auth/logout",
    method: ["DELETE"],
    body: {
      code: "00000",
      data: {},
      msg: "string",
    },
  },
]);
