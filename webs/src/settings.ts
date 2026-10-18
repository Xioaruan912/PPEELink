import { SizeEnum } from "./enums/SizeEnum";
import { ThemeEnum } from "./enums/ThemeEnum";
import { LanguageEnum } from "./enums/LanguageEnum";

const { pkg } = __APP_INFO__;

const defaultSettings: AppSettings = {
  title: "PPEELink",
  version: pkg.version,
  theme: ThemeEnum.LIGHT,
  size: SizeEnum.DEFAULT,
  language: LanguageEnum.ZH_CN,
};

export default defaultSettings;
