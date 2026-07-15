export class Box {
  helper() {
    return 1;
  }
  stay() {
    return 2;
  }
}
export function use() {
  return new Box().helper() + (new Box()).helper() + new Box().stay();
}
export function typed(b) {
  return b.helper();
}
