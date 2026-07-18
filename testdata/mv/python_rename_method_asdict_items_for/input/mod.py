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
    # homogeneous field values — items() value slot is safe
    a: A
    c: A


@dataclass
class Box:
    # mixed field values — for-in .items() must fail closed
    a: A
    b: B


def use_items(pair: Pair) -> int:
    total = 0
    for k, x in asdict(pair).items():
        total += x.run()
    return total


def use_dc_items(pair: Pair) -> int:
    total = 0
    for k, x in dataclasses.asdict(pair).items():
        total += x.run()
    return total


def use_vars_items(pair: Pair) -> int:
    total = 0
    for k, x in vars(pair).items():
        total += x.run()
    return total


def use_dunder_items(pair: Pair) -> int:
    total = 0
    for k, x in pair.__dict__.items():
        total += x.run()
    return total


def use_assigned(pair: Pair) -> int:
    d = asdict(pair)
    total = 0
    for k, x in d.items():
        total += x.run()
    return total


def use_assigned_vars(pair: Pair) -> int:
    d = vars(pair)
    total = 0
    for k, x in d.items():
        total += x.run()
    return total


def use_walrus(pair: Pair) -> int:
    total = 0
    if (d := asdict(pair)):
        for k, x in d.items():
            total += x.run()
    return total


def use_comp(pair: Pair) -> list[int]:
    return [x.run() for k, x in asdict(pair).items()]


def use_mixed_fail_closed(box: Box) -> int:
    # mixed A/B values — leave receivers unbound (fail closed)
    total = 0
    for k, item in asdict(box).items():
        total += item.run()
    return total


def use_mixed_key_still(box: Box) -> int:
    # key path still renames A; B stays put
    return asdict(box)["a"].run() + asdict(box)["b"].run()
