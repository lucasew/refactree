export class Box {
  static assist = 1;
  static stay = 2;

  use() {
    return Box.assist + Box.stay;
  }
}
