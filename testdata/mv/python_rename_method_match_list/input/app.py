class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_match_single(items: list[A]):
    match items:
        case [a]:
            return a.run()


def use_match_single_b(items: list[B]):
    match items:
        case [b]:
            return b.run()


def use_match_star(items: list[A]):
    match items:
        case [a, *rest]:
            return a.run()


def use_match_star_b(items: list[B]):
    match items:
        case [b, *rest]:
            return b.run()


def use_match_star_rest(items: list[A]):
    match items:
        case [a, *rest]:
            a.run()
            for x in rest:
                x.run()


def use_match_assigned():
    xs = [A()]
    match xs:
        case [a]:
            return a.run()
    ys = [B()]
    match ys:
        case [b]:
            return b.run()


def use_match_literal():
    match [A(), A()]:
        case [a, *rest]:
            a.run()
            for x in rest:
                x.run()
    match [B(), B()]:
        case [b, *tail]:
            return b.run()
