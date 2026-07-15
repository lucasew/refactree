export const api = {
  helper() { return 1 },
  stay() { return 2 },
}
export function use() {
  return api.helper() + api.stay();
}
