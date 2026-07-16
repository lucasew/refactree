class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_for_literal():
    for a in [A()]:
        a.execute()


def use_for_literal_b():
    for b in [B()]:
        b.run()


def use_for_annotated(items: list[A]):
    for a in items:
        a.execute()


def use_for_annotated_b(items: list[B]):
    for b in items:
        b.run()


def use_for_assigned():
    xs: list[A] = []
    for a in xs:
        a.execute()
    ys = [A()]
    for a in ys:
        a.execute()


def use_walrus():
    if (a := A()):
        a.execute()
    if (b := B()):
        b.run()


def use_comprehension():
    return [a.execute() for a in [A()]] + [b.run() for b in [B()]]
