export function assist() {
  return 1;
}

export function stay() {
  return 2;
}

export class Box {
  static {
    this.x = assist() + stay();
  }
}
