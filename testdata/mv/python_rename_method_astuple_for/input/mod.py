from dataclasses import dataclass, astuple, replace
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
    # mixed field values — for-in astuple must fail closed
    a: A
    b: B


def use_astuple(pair: Pair) -> int:
    total = 0
    for x in astuple(pair):
        total += x.run()
    return total


def use_dc_astuple(pair: Pair) -> int:
    total = 0
    for x in dataclasses.astuple(pair):
        total += x.run()
    return total


def use_list_astuple(pair: Pair) -> int:
    total = 0
    for x in list(astuple(pair)):
        total += x.run()
    return total


def use_tuple_astuple(pair: Pair) -> int:
    total = 0
    for x in tuple(astuple(pair)):
        total += x.run()
    return total


def use_assigned(pair: Pair) -> int:
    t = astuple(pair)
    total = 0
    for x in t:
        total += x.run()
    return total


def use_list_assigned(pair: Pair) -> int:
    xs = list(astuple(pair))
    total = 0
    for x in xs:
        total += x.run()
    return total


def use_replace(pair: Pair) -> int:
    total = 0
    for x in astuple(replace(pair)):
        total += x.run()
    return total


def use_walrus(pair: Pair) -> int:
    total = 0
    if (t := astuple(pair)):
        for x in t:
            total += x.run()
    return total


def use_comp(pair: Pair) -> list[int]:
    return [x.run() for x in astuple(pair)]


def use_mixed_fail_closed(box: Box) -> int:
    # mixed A/B values — leave receivers unbound (fail closed)
    total = 0
    for item in astuple(box):
        total += item.run()
    return total


def use_mixed_index_still(box: Box) -> int:
    # index path still renames A; B stays put
    return astuple(box)[0].run() + astuple(box)[1].run()
