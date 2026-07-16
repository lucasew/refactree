class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_for_literal():
    for a in [A()]:
        a.run()


def use_for_literal_b():
    for b in [B()]:
        b.run()


def use_for_annotated(items: list[A]):
    for a in items:
        a.run()


def use_for_annotated_b(items: list[B]):
    for b in items:
        b.run()


def use_for_assigned():
    xs: list[A] = []
    for a in xs:
        a.run()
    ys = [A()]
    for a in ys:
        a.run()


def use_walrus():
    if (a := A()):
        a.run()
    if (b := B()):
        b.run()


def use_comprehension():
    return [a.run() for a in [A()]] + [b.run() for b in [B()]]
