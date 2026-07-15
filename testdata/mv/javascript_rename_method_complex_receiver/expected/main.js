export class Box {
  assist() { return 1 }
  stay() { return 2 }
}
export async function make() { return new Box() }
export async function use(b, flag) {
  return (await make()).assist()
    + (flag ? b : new Box()).assist()
    + (b ?? new Box()).assist()
    + (b ?? new Box()).stay()
}
