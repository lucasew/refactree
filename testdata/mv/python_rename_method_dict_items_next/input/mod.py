class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_next_iter_items(d: dict[str, A]) -> int:
    k, a = next(iter(d.items()))
    return a.run()


def use_next_items(d: dict[str, A]) -> int:
    k, a = next(d.items())
    return a.run()


def use_paren(d: dict[str, A]) -> int:
    (k, a) = next(iter(d.items()))
    return a.run()


def use_list_pattern(d: dict[str, A]) -> int:
    [k, a] = next(iter(d.items()))
    return a.run()


def use_assigned() -> int:
    d: dict[str, A] = {}
    k, a = next(iter(d.items()))
    return a.run()


def use_b(d: dict[str, B]) -> int:
    k, b = next(iter(d.items()))
    return b.run()
