from dataclasses import dataclass, replace
import copy
import dataclasses


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


@dataclass
class Box:
    a: A
    b: B


def use_field_direct(box: Box) -> int:
    return copy.copy(box.a).run() + copy.copy(box.b).run()


def use_field_assign(box: Box) -> int:
    xa = copy.copy(box.a)
    xb = copy.copy(box.b)
    return xa.run() + xb.run()


def use_deepcopy_field(box: Box) -> int:
    return copy.deepcopy(box.a).run() + copy.deepcopy(box.b).run()


def use_replace_field(box: Box) -> int:
    return copy.copy(replace(box).a).run() + copy.copy(dataclasses.replace(box).b).run()


def use_item_direct(item: A, other: B) -> int:
    return copy.copy(item).run() + copy.copy(other).run()


def use_item_assign(item: A, other: B) -> int:
    a = copy.copy(item)
    b = copy.copy(other)
    return a.run() + b.run()


def use_import_copy(box: Box, item: A, other: B) -> int:
    from copy import copy, deepcopy
    return copy(box.a).run() + deepcopy(box.b).run() + copy(item).run() + deepcopy(other).run()
