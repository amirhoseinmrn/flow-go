
#ifndef _REL_DKG_INCLUDE_H
#define _REL_DKG_INCLUDE_H

#include "relic.h"
#include "misc.h"

#define MAX_IND         255
#define MAX_IND_BITS    8

void Zr_polynomialImage(byte* out, ep2_st* y, const bn_st* a, const int a_size, const int x);
void G2_polynomialImages(ep2_st* y, const int len_y, const ep2_st* A, const int len_A);
void ep2_vector_write_bin(byte* out, const ep2_st* A, const int len);
void ep2_vector_read_bin(ep2_st* A, const byte* src, const int len);
int verifyshare(const bn_st* x, const ep2_st* y);

#endif