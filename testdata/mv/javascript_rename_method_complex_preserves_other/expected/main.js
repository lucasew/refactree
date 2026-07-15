export class Box {
  assist() { return 1 }
}
export class Other {
  helper() { return 9 }
}
export function use(b, o, f) {
  return (b ?? new Box()).helper()
    + (f ? o : o).helper()
    + Other.helper()
}
