export async function use() {
  const { helper, stay } = await import("./box.js");
  return helper() + stay();
}
