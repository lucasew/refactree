export const api = {
  assist() { return 1 },
  stay() { return 2 },
}
export function use() {
  return api.assist() + api.stay();
}
