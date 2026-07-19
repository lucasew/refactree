class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_star_head(items: list[A]):
    a, *rest = items
    a.execute()


def use_star_head_b(items: list[B]):
    b, *rest = items
    b.run()


def use_star_tail(items: list[A]):
    *rest, a = items
    a.execute()


def use_star_tail_b(items: list[B]):
    *rest, b = items
    b.run()


def use_single_comma(items: list[A]):
    a, = items
    a.execute()


def use_single_comma_b(items: list[B]):
    b, = items
    b.run()


def use_star_mid(items: list[A]):
    a, *rest, c = items
    a.execute()
    c.execute()


def use_star_assigned():
    xs = [A()]
    a, *rest = xs
    a.execute()
    ys = [B()]
    b, *rest = ys
    b.run()


def use_star_literal():
    a, *rest = [A(), A()]
    a.execute()
    b, *rest = [B(), B()]
    b.run()


def use_star_rest_loop(items: list[A]):
    a, *rest = items
    a.execute()
    for x in rest:
        x.execute()
