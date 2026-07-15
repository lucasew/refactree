export class Box {
  static helper = 1;
  static stay = 2;

  use() {
    return Box.helper + Box.stay;
  }
}
