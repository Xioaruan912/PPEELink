/** Copy the full value; the caller controls masking and success feedback. */
export async function copySubscriptionText(value: string) {
  if (!value) throw new Error("链接为空");
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(value);
    return;
  }
  const input = document.createElement("textarea");
  const previous = document.activeElement as HTMLElement | null;
  input.value = value;
  input.style.position = "fixed";
  input.style.opacity = "0";
  document.body.appendChild(input);
  try {
    input.select();
    if (!document.execCommand("copy")) throw new Error("复制失败");
  } finally {
    input.remove();
    previous?.focus();
  }
}

export function maskSubscriptionLink(value: string) {
  if (/(token=)([^&]+)/i.test(value))
    return value.replace(
      /(token=)([^&]+)/gi,
      (_, key, token) => key + token.slice(0, 4) + "••••••••" + token.slice(-4)
    );
  // Import schemes may encode the whole subscription URL, including its token.
  return value.length > 24 ? value.slice(0, 16) + "••••••••" : "••••••••";
}
