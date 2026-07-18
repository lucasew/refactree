class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_enum_sub(items: list[A]):
    for item in enumerate(items):
        a = item[1]
        a.run()


def use_enum_sub_paren(items: list[A]):
    for item in enumerate(items):
        a = (item)[1]
        a.run()


def use_enum_sub_walrus(items: list[A]):
    for item in enumerate(items):
        if (a := item[1]):
            a.run()


def use_enum_sub_b(items: list[B]):
    for item in enumerate(items):
        b = item[1]
        b.run()


def use_enum_sub_literal():
    for item in enumerate([A()]):
        a = item[1]
        a.run()
    for item in enumerate([B()]):
        b = item[1]
        b.run()


def use_enum_sub_assigned():
    xs = [A()]
    for item in enumerate(xs):
        a = item[1]
        a.run()
    ys = [B()]
    for item in enumerate(ys):
        b = item[1]
        b.run()


def use_enum_sub_zip(xs: list[A], ys: list[A]):
    for pair in zip(xs, ys):
        a = pair[0]
        a.run()
        c = pair[1]
        c.run()


def use_enum_sub_zip_b(xs: list[B], ys: list[B]):
    for pair in zip(xs, ys):
        b = pair[0]
        b.run()


def use_enum_sub_preserves_b(items: list[B]):
    for item in enumerate(items):
        b = item[1]
        b.run()
