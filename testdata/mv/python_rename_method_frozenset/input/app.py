class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_frozenset_for(items: list[A]):
    for a in frozenset(items):
        a.run()


def use_frozenset_for_b(items: list[B]):
    for b in frozenset(items):
        b.run()


def use_frozenset_assign(items: list[A]):
    s = frozenset(items)
    for a in s:
        a.run()


def use_frozenset_assign_b(items: list[B]):
    s = frozenset(items)
    for b in s:
        b.run()


def use_frozenset_literal():
    for a in frozenset([A()]):
        a.run()
    for b in frozenset([B()]):
        b.run()


def use_frozenset_nested(items: list[A]):
    for a in list(frozenset(items)):
        a.run()


def use_frozenset_assigned_literal():
    xs = [A()]
    for a in frozenset(xs):
        a.run()
    ys = [B()]
    for b in frozenset(ys):
        b.run()
