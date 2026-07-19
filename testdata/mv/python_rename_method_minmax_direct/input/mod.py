from dataclasses import dataclass, asdict, astuple
import dataclasses
from random import choice
import random


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


@dataclass
class Pair:
    # homogeneous field values — min/max over values is safe
    a: A
    c: A


@dataclass
class Box:
    # mixed field values — min/max over values must fail closed
    a: A
    b: B


def use_min_values_direct(pair: Pair) -> int:
    return min(asdict(pair).values(), key=lambda v: 0).run()


def use_max_values_direct(pair: Pair) -> int:
    return max(asdict(pair).values(), key=lambda v: 0).run()


def use_min_no_key_direct(pair: Pair) -> int:
    return min(asdict(pair).values()).run()


def use_dc_direct(pair: Pair) -> int:
    return min(dataclasses.asdict(pair).values(), key=lambda v: 0).run()


def use_vars_direct(pair: Pair) -> int:
    return min(vars(pair).values(), key=lambda v: 0).run()


def use_dunder_direct(pair: Pair) -> int:
    return min(pair.__dict__.values(), key=lambda v: 0).run()


def use_list_values_direct(pair: Pair) -> int:
    return min(list(asdict(pair).values()), key=lambda v: 0).run()


def use_astuple_direct(pair: Pair) -> int:
    return min(astuple(pair), key=lambda v: 0).run()


def use_assigned_values_direct(pair: Pair) -> int:
    d = asdict(pair)
    return min(d.values(), key=lambda v: 0).run()


def use_min_items_direct(items: list[A]) -> int:
    return min(items).run()


def use_max_items_direct(xs: list[A]) -> int:
    return max(xs, key=lambda x: 0).run()


def use_choice_direct(ys: list[A]) -> int:
    return choice(ys).run()


def use_random_choice_direct(zs: list[A]) -> int:
    return random.choice(zs).run()


def use_min_dict_values_direct(d: dict[str, A]) -> int:
    return min(d.values(), key=lambda v: 0).run()


def use_min_assign(pair: Pair) -> int:
    # assignment path still works (regression)
    x = min(asdict(pair).values(), key=lambda v: 0)
    return x.run()


def use_mixed_fail_closed(box: Box) -> int:
    # mixed A/B — leave receiver unbound (fail closed)
    return min(asdict(box).values(), key=lambda v: 0).run()


def use_b_min(others: list[B]) -> int:
    return min(others).run()


def use_b_choice(bobs: list[B]) -> int:
    return choice(bobs).run()


def use_key_still(box: Box) -> int:
    # key path still renames A; B stays put
    return asdict(box)["a"].run() + asdict(box)["b"].run()
