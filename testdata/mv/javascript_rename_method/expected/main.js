export class Box {
  fetchValue() {
    return 1;
  }

  twice() {
    return this.fetchValue() + Box.fetchValue.call(this);
  }
}

export function use(b) {
  return b.fetchValue();
}

export function make() {
  const box = new Box();
  return box.fetchValue();
}
