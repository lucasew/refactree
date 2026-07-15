export class Crate {
  constructor(n) {
    this.n = n;
  }
  helper() {
    return this.n;
  }
}
export function main() {
  return new Crate(1).n + new Crate(2).helper();
}
export class Stay {
  constructor() {}
}
export function other() {
  return new Stay();
}
