from dataclasses import dataclass, asdict
import dataclasses


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


@dataclass
class Pair:
    a: A
    c: A


@dataclass
class Box:
    a: A
    b: B


def use_next_iter_items(pair: Pair) -> int:
    k, x = next(iter(asdict(pair).items()))
    return x.execute()


def use_next_items(pair: Pair) -> int:
    k, x = next(asdict(pair).items())
    return x.execute()


def use_dc(pair: Pair) -> int:
    k, x = next(iter(dataclasses.asdict(pair).items()))
    return x.execute()


def use_vars(pair: Pair) -> int:
    k, x = next(iter(vars(pair).items()))
    return x.execute()


def use_dunder(pair: Pair) -> int:
    k, x = next(iter(pair.__dict__.items()))
    return x.execute()


def use_assigned(pair: Pair) -> int:
    d = asdict(pair)
    k, x = next(iter(d.items()))
    return x.execute()


def use_paren(pair: Pair) -> int:
    (k, x) = next(iter(asdict(pair).items()))
    return x.execute()


def use_list_pattern(pair: Pair) -> int:
    [k, x] = next(iter(asdict(pair).items()))
    return x.execute()


def use_mixed_fail_closed(box: Box) -> int:
    k, item = next(iter(asdict(box).items()))
    return item.run()


def use_key_still(box: Box) -> int:
    return asdict(box)["a"].execute() + asdict(box)["b"].run()
