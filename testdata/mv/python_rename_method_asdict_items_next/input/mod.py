from dataclasses import dataclass, asdict
import dataclasses


class A:
    def run(self) -> int:
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
    return x.run()


def use_next_items(pair: Pair) -> int:
    k, x = next(asdict(pair).items())
    return x.run()


def use_dc(pair: Pair) -> int:
    k, x = next(iter(dataclasses.asdict(pair).items()))
    return x.run()


def use_vars(pair: Pair) -> int:
    k, x = next(iter(vars(pair).items()))
    return x.run()


def use_dunder(pair: Pair) -> int:
    k, x = next(iter(pair.__dict__.items()))
    return x.run()


def use_assigned(pair: Pair) -> int:
    d = asdict(pair)
    k, x = next(iter(d.items()))
    return x.run()


def use_paren(pair: Pair) -> int:
    (k, x) = next(iter(asdict(pair).items()))
    return x.run()


def use_list_pattern(pair: Pair) -> int:
    [k, x] = next(iter(asdict(pair).items()))
    return x.run()


def use_min_items(pair: Pair) -> int:
    k, x = min(asdict(pair).items(), key=lambda kv: 0)
    return x.run()


def use_next_sub(pair: Pair) -> int:
    x = next(iter(asdict(pair).items()))[1]
    return x.run()


def use_pair_local(pair: Pair) -> int:
    p = next(iter(asdict(pair).items()))
    k, x = p
    return x.run()




def use_mixed_fail_closed(box: Box) -> int:
    k, item = next(iter(asdict(box).items()))
    return item.run()


def use_key_still(box: Box) -> int:
    return asdict(box)["a"].run() + asdict(box)["b"].run()
