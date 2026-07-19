class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_list_append():
    xs = []
    ys = []
    xs.append(A())
    ys.append(B())
    return xs[0].run() + ys[0].run()


def use_list_append_for():
    xs = []
    ys = []
    xs.append(A())
    ys.append(B())
    n = 0
    for a in xs:
        n += a.run()
    for b in ys:
        n += b.run()
    return n


def use_list_append_var():
    xs = []
    ys = []
    xs.append(A())
    ys.append(B())
    a = xs[0]
    b = ys[0]
    return a.run() + b.run()


def use_list_ctor_append():
    xs = list()
    ys = list()
    xs.append(A())
    ys.append(B())
    return xs[0].run() + ys[0].run()


def use_preserves_b():
    ys = []
    ys.append(B())
    return ys[0].run()
