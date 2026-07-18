class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_list_zip_index_unpack(xs: list[A], ys: list[A]):
    a, b = list(zip(xs, ys))[0]
    a.execute()
    b.execute()


def use_pairs_index_unpack(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a, b = pairs[0]
    a.execute()
    b.execute()


def use_pairs_double_sub(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a = pairs[0][0]
    a.execute()
    b = pairs[0][1]
    b.execute()


def use_list_zip_double_sub(xs: list[A], ys: list[A]):
    a = list(zip(xs, ys))[0][0]
    a.execute()


def use_pair_from_index(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    pair = pairs[0]
    a = pair[0]
    a.execute()
    c = pair[1]
    c.execute()


def use_pair_from_index_unpack(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    pair = pairs[0]
    a, b = pair
    a.execute()
    b.execute()


def use_walrus_index_pair(xs: list[A], ys: list[A]):
    if (pair := list(zip(xs, ys))[0]):
        a, b = pair
        a.execute()
        b.execute()


def use_tuple_zip_index(xs: list[A], ys: list[A]):
    a, b = tuple(zip(xs, ys))[0]
    a.execute()
    b.execute()


def use_list_zip_index_literal():
    a, b = list(zip([A()], [A()]))[0]
    a.execute()
    x, y = list(zip([B()], [B()]))[0]
    x.run()


def use_list_zip_index_b(xs: list[B], ys: list[B]):
    x, y = list(zip(xs, ys))[0]
    x.run()


def use_pairs_double_sub_b(xs: list[B], ys: list[B]):
    pairs = list(zip(xs, ys))
    x = pairs[0][0]
    x.run()


def use_list_zip_index_preserves_b(xs: list[B], ys: list[B]):
    pairs = list(zip(xs, ys))
    pair = pairs[0]
    x, y = pair
    x.run()
