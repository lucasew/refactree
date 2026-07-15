export class Box {
  helper() { return 1 }
  stay() { return 2 }
}
export function use(a, b, f) {
  return (b ?? new Box()).helper()
    + (f ? a : b).helper()
    + (b).helper()
    + b.stay()
}
