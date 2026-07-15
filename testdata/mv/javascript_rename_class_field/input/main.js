export class Box {
  helper = 1;
  stay = 2;

  use() {
    return this.helper + this.stay;
  }
}

export function run(b) {
  return b.helper + b.stay;
}
