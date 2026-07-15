export class Box {
  helper() { return 1 }
  stay() { return 2 }
}
export async function make() { return new Box() }
export async function use(b, flag) {
  return (await make()).helper()
    + (flag ? b : new Box()).helper()
    + (b ?? new Box()).helper()
    + (b ?? new Box()).stay()
}
