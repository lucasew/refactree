export type Worker = {
  assist(): number;
  stay(): number;
};
export function use(w: Worker): number {
  return w.assist() + w.stay();
}
