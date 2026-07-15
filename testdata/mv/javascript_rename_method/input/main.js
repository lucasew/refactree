export class Box {
  getValue() {
    return 1;
  }

  twice() {
    return this.getValue() + Box.getValue.call(this);
  }
}

export function use(b) {
  return b.getValue();
}

export function make() {
  const box = new Box();
  return box.getValue();
}
