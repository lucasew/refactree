export async function use() {
  const m = await import("./box.js");
  return m.helper() + m.stay();
}
