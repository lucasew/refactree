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
export class Other {
  helper(): number {
    return 9;
  }
}
export function use(w: Worker, o: Other): number {
  return w.assist() + o.helper() + w.stay();
}
