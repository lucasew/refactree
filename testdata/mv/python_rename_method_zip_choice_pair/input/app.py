import heapq
import operator
import random
from heapq import heappop
from operator import itemgetter
from random import choice


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_choice_unpack(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a, b = choice(pairs)
    a.run()
    b.run()


def use_random_choice_unpack(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a, b = random.choice(pairs)
    a.run()
    b.run()


def use_choice_pair_sub(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    pair = choice(pairs)
    a = pair[0]
    a.run()
    c = pair[1]
    c.run()


def use_choice_pair_unpack(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    pair = random.choice(pairs)
    a, b = pair
    a.run()
    b.run()


def use_choice_nested(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    pair = choice(pairs)
    for a in pair:
        a.run()
    b = next(iter(pair))
    b.run()


def use_choice_sub_direct(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a = choice(pairs)[0]
    a.run()
    b = random.choice(pairs)[1]
    b.run()


def use_list_zip_choice_unpack(xs: list[A], ys: list[A]):
    a, b = choice(list(zip(xs, ys)))
    a.run()
    b.run()


def use_walrus_choice_pair(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    if (pair := choice(pairs)):
        a, b = pair
        a.run()
        b.run()
        for c in pair:
            c.run()


def use_heappop_unpack(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a, b = heappop(pairs)
    a.run()
    b.run()


def use_heapq_heappop_pair(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    pair = heapq.heappop(pairs)
    a = pair[0]
    a.run()
    b, c = pair
    b.run()
    c.run()


def use_heappop_sub_direct(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a = heappop(pairs)[0]
    a.run()
    b = heapq.heappop(pairs)[1]
    b.run()


def use_heappop_nested(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    pair = heappop(pairs)
    for a in pair:
        a.run()


def use_itemgetter_unpack(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a, b = itemgetter(0)(pairs)
    a.run()
    b.run()


def use_operator_itemgetter_pair(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    pair = operator.itemgetter(0)(pairs)
    a = pair[0]
    a.run()
    b, c = pair
    b.run()
    c.run()


def use_itemgetter_sub_direct(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a = itemgetter(0)(pairs)[0]
    a.run()
    b = operator.itemgetter(0)(pairs)[1]
    b.run()


def use_itemgetter_nested(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    pair = itemgetter(0)(pairs)
    for a in pair:
        a.run()
    b = next(iter(pair))
    b.run()


def use_list_zip_itemgetter(xs: list[A], ys: list[A]):
    a, b = itemgetter(0)(list(zip(xs, ys)))
    a.run()
    b.run()


def use_walrus_itemgetter_pair(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    if (pair := itemgetter(0)(pairs)):
        a, b = pair
        a.run()
        b.run()


def use_choice_literal():
    pairs = list(zip([A()], [A()]))
    a, b = choice(pairs)
    a.run()
    pairs2 = list(zip([B()], [B()]))
    x, y = choice(pairs2)
    x.run()


def use_choice_unpack_b(xs: list[B], ys: list[B]):
    pairs = list(zip(xs, ys))
    x, y = choice(pairs)
    x.run()


def use_heappop_pair_sub_b(xs: list[B], ys: list[B]):
    pairs = list(zip(xs, ys))
    pair = heappop(pairs)
    x = pair[0]
    x.run()


def use_itemgetter_nested_b(xs: list[B], ys: list[B]):
    pairs = list(zip(xs, ys))
    pair = itemgetter(0)(pairs)
    for x in pair:
        x.run()


def use_choice_preserves_b(xs: list[B], ys: list[B]):
    pairs = list(zip(xs, ys))
    pair = random.choice(pairs)
    x, y = pair
    x.run()
