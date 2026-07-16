class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_enumerate(items: list[A]):
    for i, a in enumerate(items):
        a.execute()


def use_enumerate_b(items: list[B]):
    for i, b in enumerate(items):
        b.run()


def use_enumerate_literal():
    for i, a in enumerate([A()]):
        a.execute()


def use_enumerate_assigned():
    xs = [A()]
    for i, a in enumerate(xs):
        a.execute()
    ys = [B()]
    for i, b in enumerate(ys):
        b.run()


def use_zip(xs: list[A], ys: list[B]):
    for a, b in zip(xs, ys):
        a.execute()
        b.run()


def use_zip_literal():
    for a, b in zip([A()], [B()]):
        a.execute()
        b.run()


def use_zip_assigned():
    xs = [A()]
    ys = [B()]
    for a, b in zip(xs, ys):
        a.execute()
        b.run()
