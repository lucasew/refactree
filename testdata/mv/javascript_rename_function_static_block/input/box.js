export function helper() {
  return 1;
}

export function stay() {
  return 2;
}

export class Box {
  static {
    this.x = helper() + stay();
  }
}
