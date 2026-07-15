export class Box {
  assist = 1;
  stay = 2;

  use() {
    return this.assist + this.stay;
  }
}

export function run(b) {
  return b.assist + b.stay;
}
