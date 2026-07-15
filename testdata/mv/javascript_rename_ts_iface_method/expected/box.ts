export interface Worker {
  assist(): number;
  stay(): number;
}
export class Box implements Worker {
  assist(): number {
    return 1;
  }
  stay(): number {
    return 2;
  }
}
export function use(w: Worker): number {
  return w.assist() + w.stay();
}
export function useBox(b: Box): number {
  return b.assist() + b.stay();
}
