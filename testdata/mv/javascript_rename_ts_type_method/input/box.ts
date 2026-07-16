export type Worker = {
  helper(): number;
  stay(): number;
};
export function use(w: Worker): number {
  return w.helper() + w.stay();
}
