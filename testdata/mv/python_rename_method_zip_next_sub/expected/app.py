class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_next_sub(xs: list[A], ys: list[A]):
    pairs = zip(xs, ys)
    a = next(pairs)[0]
    a.execute()
    b = next(pairs)[1]
    b.execute()


def use_next_sub_direct(xs: list[A], ys: list[A]):
    a = next(zip(xs, ys))[0]
    a.execute()


def use_next_pair(xs: list[A], ys: list[A]):
    pairs = zip(xs, ys)
    pair = next(pairs)
    a = pair[0]
    a.execute()
    c = pair[1]
    c.execute()


def use_list_zip(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    for a, b in pairs:
        a.execute()
        b.execute()


def use_list_zip_nested(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    for pair in pairs:
        for a in pair:
            a.execute()


def use_next_sub_b(xs: list[B], ys: list[B]):
    pairs = zip(xs, ys)
    x = next(pairs)[0]
    x.run()


def use_list_zip_b(xs: list[B], ys: list[B]):
    pairs = list(zip(xs, ys))
    for x, y in pairs:
        x.run()


def use_next_sub_literal():
    a = next(zip([A()], [A()]))[0]
    a.execute()
    x = next(zip([B()], [B()]))[0]
    x.run()


def use_list_zip_assigned():
    xs = [A()]
    ys = [A()]
    pairs = list(zip(xs, ys))
    for a, b in pairs:
        a.execute()
    zs = [B()]
    ws = [B()]
    pairs2 = list(zip(zs, ws))
    for x, y in pairs2:
        x.run()


def use_next_sub_preserves_b(xs: list[B], ys: list[B]):
    pairs = zip(xs, ys)
    pair = next(pairs)
    x = pair[0]
    x.run()
