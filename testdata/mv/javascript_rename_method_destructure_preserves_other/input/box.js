export class Box {
  helper() {
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
  const { helper } = b;
  return helper() + b.helper();
}

export function useStay() {
  const s = new Stay();
  const { helper } = s;
  return helper() + s.helper();
}
