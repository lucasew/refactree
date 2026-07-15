export class Box {
  constructor(n) {
    this.n = n;
  }
  helper() {
    return this.n;
  }
}
export function main() {
  return new Box(1).n + new Box(2).helper();
}
export class Stay {
  constructor() {}
}
export function other() {
  return new Stay();
}
