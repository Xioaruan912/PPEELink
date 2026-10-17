import { SizeEnum } from "./enums/SizeEnum";
import { LayoutEnum } from "./enums/LayoutEnum";
import { ThemeEnum } from "./enums/ThemeEnum";
import { LanguageEnum } from "./enums/LanguageEnum";

const { pkg } = __APP_INFO__;

const defaultSettings: AppSettings = {
  title: "PPEELink",
  version: pkg.version,
  showSettings: false,
  tagsView: false,
  fixedHeader: true,
  sidebarLogo: true,
  layout: LayoutEnum.LEFT,
  theme: ThemeEnum.LIGHT,
  size: SizeEnum.DEFAULT,
  language: LanguageEnum.ZH_CN,
  themeColor: "var(--c-signal)",
  watermarkEnabled: false,
  watermarkContent: "PPEELink",
};

export default defaultSettings;
