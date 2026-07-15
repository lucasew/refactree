export const Box = class {
  assist() {
    return 1;
  }

  stay() {
    return 2;
  }

  use() {
    return this.assist() + this.stay();
  }
};

export function run(b) {
  return b.assist() + b.stay();
}
