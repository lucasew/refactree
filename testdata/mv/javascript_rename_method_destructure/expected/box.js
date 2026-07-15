export class Box {
  assist() {
    return 1;
  }

  stay() {
    return 2;
  }
}

export function use(b) {
  const { assist, stay } = b;
  return assist() + stay();
}
