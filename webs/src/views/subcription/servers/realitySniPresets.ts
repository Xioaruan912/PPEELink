export type SNIRegionKey =
  | "US"
  | "CA"
  | "JP"
  | "HK"
  | "SG"
  | "TW"
  | "KR"
  | "EU"
  | "GB"
  | "GLOBAL";

export interface SNIRegionGroup {
  key: SNIRegionKey;
  label: string;
  hosts: string[];
}

/** 实测验证（TLS 1.3 + HTTP/2 + 非 Cloudflare CDN）的各地区本土 Reality 伪装站 */
export const REGIONAL_SNI_GROUPS: SNIRegionGroup[] = [
  {
    key: "JP",
    label: "日本",
    hosts: [
      "www.sony.jp",
      "www.yahoo.co.jp",
      "www.honda.co.jp",
      "www.yamaha.com",
      "www.lovelive-anime.jp",
    ],
  },
  {
    key: "HK",
    label: "香港",
    hosts: [
      "www.mandarinoriental.com",
      "www.k11musea.com",
      "www.pacificplace.com.hk",
      "www.hko.gov.hk",
      "www.hku.hk",
    ],
  },
  {
    key: "US",
    label: "美国",
    hosts: [
      "www.ebay.com",
      "www.airbnb.com",
      "www.ups.com",
      "www.stanford.edu",
      "www.harvard.edu",
    ],
  },
  {
    key: "SG",
    label: "新加坡",
    hosts: [
      "www.grab.com",
      "www.changiairport.com",
      "nus.edu.sg",
      "www.smu.edu.sg",
      "www.sutd.edu.sg",
    ],
  },
  {
    key: "TW",
    label: "台湾",
    hosts: [
      "www.asus.com",
      "www.msi.com",
      "www.gigabyte.com",
      "www.evaair.com",
      "www.gskill.com",
    ],
  },
  {
    key: "KR",
    label: "韩国",
    hosts: [
      "www.naver.com",
      "www.samsung.com",
      "www.koreanair.com",
      "www.coupang.com",
      "www.cj.net",
    ],
  },
  {
    key: "CA",
    label: "加拿大",
    hosts: [
      "www.utoronto.ca",
      "www.ubc.ca",
      "www.ebay.com",
      "www.airbnb.com",
      "www.ups.com",
    ],
  },
  {
    key: "EU",
    label: "欧洲",
    hosts: [
      "www.adidas.com",
      "www.siemens.com",
      "www.mercedes-benz.com",
      "www.tudelft.nl",
      "www.postgresql.org",
    ],
  },
  {
    key: "GB",
    label: "英国",
    hosts: [
      "www.virginatlantic.com",
      "www.marksandspencer.com",
      "www.unilever.com",
      "www.arm.com",
      "www.cam.ac.uk",
    ],
  },
  {
    key: "GLOBAL",
    label: "全球兜底",
    hosts: [
      "www.gentoo.org",
      "www.python.org",
      "www.debian.org",
      "www.kernel.org",
      "addons.mozilla.org",
    ],
  },
];

export function inferSNIRegion(hint?: string): SNIRegionKey {
  const s = (hint || "").toUpperCase().trim();
  if (!s) return "GLOBAL";
  if (/香港|HONG\s?KONG|(_|-|\b)HK\b|HK$/.test(s)) return "HK";
  if (/日本|东京|大阪|JAPAN|TOKYO|(_|-|\b)JP\b|JP$/.test(s)) return "JP";
  if (/美国|洛杉矶|圣何塞|西雅图|USA|UNITED STATES|(_|-|\b)US\b|US$/.test(s))
    return "US";
  if (/新加坡|狮城|SINGAPORE|(_|-|\b)SG\b|SG$/.test(s)) return "SG";
  if (/台湾|台灣|台北|TAIWAN|(_|-|\b)TW\b|TW$/.test(s)) return "TW";
  if (/韩国|首尔|KOREA|SEOUL|(_|-|\b)KR\b|KR$/.test(s)) return "KR";
  if (/加拿大|多伦多|温哥华|CANADA|(_|-|\b)CA\b|CA$/.test(s)) return "CA";
  if (/英国|伦敦|LONDON|(_|-|\b)(UK|GB)\b|(UK|GB)$/.test(s)) return "GB";
  if (/德国|法国|荷兰|欧洲|法兰克福|阿姆斯特丹|(_|-|\b)(DE|FR|NL|EU)\b/.test(s))
    return "EU";
  return "GLOBAL";
}
