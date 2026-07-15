export interface Worker {
  helper(): number;
  stay(): number;
}
export class Box implements Worker {
  helper(): number {
    return 1;
  }
  stay(): number {
    return 2;
  }
}
export function use(w: Worker): number {
  return w.helper() + w.stay();
}
export function useBox(b: Box): number {
  return b.helper() + b.stay();
}
