export class Box {
  assist() { return 1 }
  stay() { return 2 }
}
export function use(a, b, f) {
  return (b ?? new Box()).assist()
    + (f ? a : b).assist()
    + (b).assist()
    + b.stay()
}
