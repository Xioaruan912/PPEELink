import { ref } from "vue";
import { getCountries, type CountryOption } from "@/api/subcription/node";

/** 国家列表（来自后端 node.Countries，常用在前），整个页面共用一份。 */
export const countries = ref<CountryOption[]>([]);
let loading: Promise<void> | null = null;
export function loadCountries() {
  if (countries.value.length) return Promise.resolve();
  loading ||= getCountries()
    .then(({ data }) => {
      countries.value = Array.isArray(data) ? data : [];
    })
    .catch(() => {
      loading = null;
    });
  return loading;
}

const FLAG_PAIR = /[\u{1F1E6}-\u{1F1FF}]{2}/u;
const isCodeAlias = (alias: string) => /^[A-Za-z]{1,3}$/.test(alias);

/**
 * 从节点名识别国家，规则与后端 node.CountryFromName 一致：取最靠前的线索，
 * 国旗 emoji、中文/英文名（不区分大小写）、或全大写的 2/3 位代码片段（JP、USA）。
 */
export function countryFromName(name: string): CountryOption | null {
  if (!name.trim() || !countries.value.length) return null;
  const byCode = new Map(countries.value.map((c) => [c.code, c]));
  const codeAliases = new Map<string, string>([["UK", "GB"]]);
  for (const c of countries.value)
    for (const a of c.aliases)
      if (/^[A-Z]{3}$/.test(a)) codeAliases.set(a, c.code);

  let best: string | null = null,
    bestAt = Infinity,
    bestLen = 0;
  const consider = (code: string, at: number, len: number) => {
    if (at >= 0 && (at < bestAt || (at === bestAt && len > bestLen))) {
      best = code;
      bestAt = at;
      bestLen = len;
    }
  };
  const flag = FLAG_PAIR.exec(name);
  if (flag) {
    const [a, b] = Array.from(flag[0]);
    const code = String.fromCharCode(
      a.codePointAt(0)! - 0x1f1e6 + 65,
      b.codePointAt(0)! - 0x1f1e6 + 65
    );
    if (byCode.has(code)) consider(code, flag.index, flag[0].length);
  }
  const lower = name.toLowerCase();
  for (const c of countries.value)
    for (const a of c.aliases)
      if (!isCodeAlias(a.replace(/ /g, "")) || a.length > 3)
        consider(c.code, lower.indexOf(a.toLowerCase()), a.length);
  for (const m of name.matchAll(/[A-Za-z]+/g)) {
    const token = m[0];
    if (token !== token.toUpperCase()) continue;
    if (token.length === 2) {
      if (token === "CN" && name.slice(m.index! + 2).startsWith("2")) continue;
      const code = codeAliases.get(token) || (byCode.has(token) ? token : "");
      if (code) consider(code, m.index!, 2);
    } else if (token.length === 3) {
      const code = codeAliases.get(token);
      if (code) consider(code, m.index!, 3);
    }
  }
  return best ? byCode.get(best) || null : null;
}

/** 去掉名称里的国旗 emoji。 */
export const stripFlags = (name: string) =>
  name.replace(new RegExp(FLAG_PAIR.source, "gu"), "").trim();
