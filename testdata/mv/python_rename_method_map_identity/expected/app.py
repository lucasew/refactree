class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_map_list(aa: list[A], bb: list[B]):
    return list(map(lambda x: x, aa))[0].execute() + list(map(lambda x: x, bb))[0].run()


def use_map_for(aa: list[A], bb: list[B]):
    s = 0
    for a in map(lambda x: x, aa):
        s += a.execute()
    for b in map(lambda x: x, bb):
        s += b.run()
    return s


def use_map_assign(aa: list[A], bb: list[B]):
    xa = list(map(lambda x: x, aa))[0]
    xb = list(map(lambda x: x, bb))[0]
    return xa.execute() + xb.run()


def use_map_paren(aa: list[A], bb: list[B]):
    return list(map(lambda x: (x), aa))[0].execute() + list(map(lambda x: (x), bb))[0].run()
