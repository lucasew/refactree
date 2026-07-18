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


def use_match_star_wild(items: list[A]):
    match items:
        case [a, *_]:
            return a.run()


def use_match_star_wild_b(items: list[B]):
    match items:
        case [b, *_]:
            return b.run()


def use_match_wild_first(items: list[A]):
    match items:
        case [_, a]:
            return a.run()


def use_match_wild_first_b(items: list[B]):
    match items:
        case [_, b]:
            return b.run()


def use_match_wild_mid(items: list[A]):
    match items:
        case [a, _, c]:
            return a.run() + c.run()


def use_match_wild_tuple(items: tuple[A, ...]):
    match items:
        case (_, a):
            return a.run()


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
    match [A(), A()]:
        case [a, *_]:
            return a.run()
    match [A(), A()]:
        case [_, a]:
            return a.run()
