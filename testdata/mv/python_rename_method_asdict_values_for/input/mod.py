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
    # homogeneous field values — iteration is safe
    a: A
    c: A


@dataclass
class Box:
    # mixed field values — for-in .values() must fail closed
    a: A
    b: B


def use_values(pair: Pair) -> int:
    total = 0
    for x in asdict(pair).values():
        total += x.run()
    return total


def use_dc_values(pair: Pair) -> int:
    total = 0
    for x in dataclasses.asdict(pair).values():
        total += x.run()
    return total


def use_vars_values(pair: Pair) -> int:
    total = 0
    for x in vars(pair).values():
        total += x.run()
    return total


def use_dunder_values(pair: Pair) -> int:
    total = 0
    for x in pair.__dict__.values():
        total += x.run()
    return total


def use_assigned(pair: Pair) -> int:
    d = asdict(pair)
    total = 0
    for x in d.values():
        total += x.run()
    return total


def use_assigned_vars(pair: Pair) -> int:
    d = vars(pair)
    total = 0
    for x in d.values():
        total += x.run()
    return total


def use_list_values(pair: Pair) -> int:
    total = 0
    for x in list(asdict(pair).values()):
        total += x.run()
    return total


def use_list_assigned(pair: Pair) -> int:
    xs = list(asdict(pair).values())
    total = 0
    for x in xs:
        total += x.run()
    return total


def use_walrus(pair: Pair) -> int:
    total = 0
    if (d := asdict(pair)):
        for x in d.values():
            total += x.run()
    return total


def use_comp(pair: Pair) -> list[int]:
    return [x.run() for x in asdict(pair).values()]


def use_mixed_fail_closed(box: Box) -> int:
    # mixed A/B values — leave receivers unbound (fail closed)
    total = 0
    for item in asdict(box).values():
        total += item.run()
    return total


def use_mixed_key_still(box: Box) -> int:
    # key path still renames A; B stays put
    return asdict(box)["a"].run() + asdict(box)["b"].run()
