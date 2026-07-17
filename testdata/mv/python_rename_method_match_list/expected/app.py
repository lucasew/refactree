class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_match_single(items: list[A]):
    match items:
        case [a]:
            return a.execute()


def use_match_single_b(items: list[B]):
    match items:
        case [b]:
            return b.run()


def use_match_star(items: list[A]):
    match items:
        case [a, *rest]:
            return a.execute()


def use_match_star_b(items: list[B]):
    match items:
        case [b, *rest]:
            return b.run()


def use_match_star_rest(items: list[A]):
    match items:
        case [a, *rest]:
            a.execute()
            for x in rest:
                x.execute()


def use_match_assigned():
    xs = [A()]
    match xs:
        case [a]:
            return a.execute()
    ys = [B()]
    match ys:
        case [b]:
            return b.run()


def use_match_literal():
    match [A(), A()]:
        case [a, *rest]:
            a.execute()
            for x in rest:
                x.execute()
    match [B(), B()]:
        case [b, *tail]:
            return b.run()
