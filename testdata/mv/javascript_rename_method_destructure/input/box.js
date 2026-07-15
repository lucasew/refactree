export class Box {
  helper() {
    return 1;
  }

  stay() {
    return 2;
  }
}

export function use(b) {
  const { helper, stay } = b;
  return helper() + stay();
}
