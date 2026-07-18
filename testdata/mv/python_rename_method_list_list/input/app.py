class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_index(aa: list[list[A]], bb: list[list[B]]) -> int:
    return aa[0][0].run() + bb[0][0].run()


def use_var(aa: list[list[A]], bb: list[list[B]]) -> int:
    ra = aa[0]
    rb = bb[0]
    return ra[0].run() + rb[0].run()


def use_for(aa: list[list[A]], bb: list[list[B]]) -> int:
    n = 0
    for row in aa:
        for a in row:
            n += a.run()
    for row in bb:
        for b in row:
            n += b.run()
    return n


def use_preserves_b(bb: list[list[B]]) -> int:
    return bb[0][0].run()
