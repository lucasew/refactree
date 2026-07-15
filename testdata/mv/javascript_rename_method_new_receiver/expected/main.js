export class Box {
  assist() {
    return 1;
  }
  stay() {
    return 2;
  }
}
export function use() {
  return new Box().assist() + (new Box()).assist() + new Box().stay();
}
export function typed(b) {
  return b.assist();
}
