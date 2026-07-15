export const Box = class {
  helper() {
    return 1;
  }

  stay() {
    return 2;
  }

  use() {
    return this.helper() + this.stay();
  }
};

export function run(b) {
  return b.helper() + b.stay();
}
