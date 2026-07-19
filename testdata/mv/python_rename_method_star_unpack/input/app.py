class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_star_head(items: list[A]):
    a, *rest = items
    a.run()


def use_star_head_b(items: list[B]):
    b, *rest = items
    b.run()


def use_star_tail(items: list[A]):
    *rest, a = items
    a.run()


def use_star_tail_b(items: list[B]):
    *rest, b = items
    b.run()


def use_single_comma(items: list[A]):
    a, = items
    a.run()


def use_single_comma_b(items: list[B]):
    b, = items
    b.run()


def use_star_mid(items: list[A]):
    a, *rest, c = items
    a.run()
    c.run()


def use_star_assigned():
    xs = [A()]
    a, *rest = xs
    a.run()
    ys = [B()]
    b, *rest = ys
    b.run()


def use_star_literal():
    a, *rest = [A(), A()]
    a.run()
    b, *rest = [B(), B()]
    b.run()


def use_star_rest_loop(items: list[A]):
    a, *rest = items
    a.run()
    for x in rest:
        x.run()
