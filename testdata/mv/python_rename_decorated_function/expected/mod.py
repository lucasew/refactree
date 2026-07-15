def deco(f):
    return f


@deco
def assist():
    return 1


def main():
    return assist()
