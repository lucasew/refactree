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


def use_next_items_sub(pair: Pair) -> int:
    p = next(iter(asdict(pair).items()))
    return p[1].execute()


def use_next_items_sub_bare(pair: Pair) -> int:
    p = next(asdict(pair).items())
    return p[1].execute()


def use_dc(pair: Pair) -> int:
    p = next(iter(dataclasses.asdict(pair).items()))
    return p[1].execute()


def use_vars(pair: Pair) -> int:
    p = next(iter(vars(pair).items()))
    return p[1].execute()


def use_dunder(pair: Pair) -> int:
    p = next(iter(pair.__dict__.items()))
    return p[1].execute()


def use_assigned(pair: Pair) -> int:
    d = asdict(pair)
    p = next(iter(d.items()))
    return p[1].execute()


def use_min_items_sub(pair: Pair) -> int:
    p = min(asdict(pair).items(), key=lambda kv: 0)
    return p[1].execute()


def use_dict_items_sub(d: dict[str, A]) -> int:
    p = next(iter(d.items()))
    return p[1].execute()


def use_paren(pair: Pair) -> int:
    p = next(iter(asdict(pair).items()))
    return (p)[1].execute()


def use_mixed_fail_closed(box: Box) -> int:
    # mixed A/B — leave receiver unbound (fail closed; distinct name so
    # file-global pairSlots from prior homogeneous p = next(...) do not shadow)
    item = next(iter(asdict(box).items()))
    return item[1].run()


def use_key_fail_closed(pair: Pair) -> int:
    # key slot [0] stays untyped — must not rename (distinct local)
    pk = next(iter(asdict(pair).items()))
    return pk[0].run()  # type: ignore[attr-defined]


def use_b_dict(d: dict[str, B]) -> int:
    # distinct name so B pairSlots does not shadow A locals above
    pb = next(iter(d.items()))
    return pb[1].run()


def use_key_still(box: Box) -> int:
    return asdict(box)["a"].execute() + asdict(box)["b"].run()
