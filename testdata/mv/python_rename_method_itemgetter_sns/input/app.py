from operator import itemgetter
import operator
from types import SimpleNamespace
import types
import copy


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_itemgetter_vars():
    return (
        itemgetter("k")(vars(SimpleNamespace(k=A()))).run()
        + itemgetter("k")(vars(SimpleNamespace(k=B()))).run()
    )


def use_itemgetter_dict():
    return (
        operator.itemgetter("k")(SimpleNamespace(k=A()).__dict__).run()
        + operator.itemgetter("k")(SimpleNamespace(k=B()).__dict__).run()
    )


def use_itemgetter_types():
    return (
        itemgetter("k")(vars(types.SimpleNamespace(k=A()))).run()
        + itemgetter("k")(vars(types.SimpleNamespace(k=B()))).run()
    )


def use_getitem_vars():
    return (
        operator.getitem(vars(SimpleNamespace(k=A())), "k").run()
        + operator.getitem(vars(SimpleNamespace(k=B())), "k").run()
    )


def use_copy_itemgetter():
    return (
        copy.copy(itemgetter("k")(vars(SimpleNamespace(k=A())))).run()
        + copy.copy(itemgetter("k")(vars(SimpleNamespace(k=B())))).run()
    )


def use_preserves_b():
    return itemgetter("k")(vars(SimpleNamespace(k=B()))).run()
