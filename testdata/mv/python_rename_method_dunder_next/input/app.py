class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_dunder_next(items: list[A]):
    it = iter(items)
    a = it.__next__()
    a.run()


def use_dunder_next_b(items: list[B]):
    it = iter(items)
    b = it.__next__()
    b.run()


def use_dunder_next_assigned():
    xs = [A()]
    it = iter(xs)
    a = it.__next__()
    a.run()
    ys = [B()]
    jt = iter(ys)
    b = jt.__next__()
    b.run()


def use_dunder_next_literal():
    it = iter([A()])
    a = it.__next__()
    a.run()
    jt = iter([B()])
    b = jt.__next__()
    b.run()


def use_dunder_next_nested(items: list[A]):
    it = iter(list(items))
    a = it.__next__()
    a.run()
