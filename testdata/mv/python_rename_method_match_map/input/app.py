class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_match_map(d: dict[str, A]):
    match d:
        case {"k": a}:
            return a.run()


def use_match_map_b(d: dict[str, B]):
    match d:
        case {"k": b}:
            return b.run()


def use_match_multi(d: dict[str, A]):
    match d:
        case {"k": a, "m": c}:
            return a.run() + c.run()


def use_match_multi_b(d: dict[str, B]):
    match d:
        case {"k": b, "m": e}:
            return b.run() + e.run()


def use_match_as(d: dict[str, A]):
    match d:
        case {"k": a as x}:
            return x.run()


def use_match_as_b(d: dict[str, B]):
    match d:
        case {"k": b as y}:
            return y.run()


def use_match_key_capture(d: dict[str, A]):
    match d:
        case {k: a}:
            return a.run()


def use_match_assigned():
    d: dict[str, A] = {}
    match d:
        case {"k": a}:
            a.run()
    e: dict[str, B] = {}
    match e:
        case {"k": b}:
            b.run()


def use_match_rest_value(d: dict[str, A]):
    match d:
        case {"k": a, **rest}:
            return a.run()
