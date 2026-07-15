export async function use() {
  const { assist, stay } = await import("./box.js");
  return assist() + stay();
}
