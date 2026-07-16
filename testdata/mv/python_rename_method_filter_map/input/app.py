class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_filter(items: list[A]):
    for a in filter(None, items):
        a.run()


def use_filter_pred(items: list[A]):
    for a in filter(lambda x: True, items):
        a.run()


def use_filter_b(items: list[B]):
    for b in filter(None, items):
        b.run()


def use_filter_assigned():
    xs = [A()]
    for a in filter(None, xs):
        a.run()
    ys = [B()]
    for b in filter(None, ys):
        b.run()


def use_filter_nested(items: list[A]):
    for a in list(filter(None, items)):
        a.run()


def use_filter_literal():
    for a in filter(None, [A()]):
        a.run()
    for b in filter(None, [B()]):
        b.run()


def use_map_ctor(names: list[str]):
    for a in map(A, names):
        a.run()


def use_map_ctor_b(names: list[str]):
    for b in map(B, names):
        b.run()
