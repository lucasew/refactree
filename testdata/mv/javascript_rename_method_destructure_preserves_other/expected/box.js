export class Box {
  assist() {
    return 1;
  }
}

export class Stay {
  helper() {
    return 2;
  }
}

export function useBox() {
  const b = new Box();
  const { assist } = b;
  return assist() + b.assist();
}

export function useStay() {
  const s = new Stay();
  const { helper } = s;
  return helper() + s.helper();
}
